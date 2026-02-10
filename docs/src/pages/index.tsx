import type {ReactNode} from 'react';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';

import styles from './index.module.css';

function HeroSection() {
  return (
    <section className={styles.hero}>
      <div className={styles.heroInner}>
        <h1 className={styles.title}>
          keylight<span className={styles.titleAccent}>d</span>
        </h1>
        <p className={styles.tagline}>
          A daemon for discovering, monitoring, and controlling Elgato Key Light
          devices. CLI, HTTP API, Unix socket, system tray, and GNOME extension
          included.
        </p>

        <div className={styles.buttons}>
          <Link className={styles.primaryBtn} to="/docs/intro">
            Get Started
          </Link>
          <Link className={styles.secondaryBtn} to="https://github.com/jmylchreest/keylightd">
            View on GitHub
          </Link>
        </div>

        <div className={styles.codePreview}>
          <div className={styles.codeHeader}>
            <span className={`${styles.codeDot} ${styles.codeDotRed}`}></span>
            <span className={`${styles.codeDot} ${styles.codeDotYellow}`}></span>
            <span className={`${styles.codeDot} ${styles.codeDotGreen}`}></span>
            <span className={styles.codeTitle}>terminal</span>
          </div>
          <div className={styles.codeContent}>
            <div className={styles.codeLine}>
              <span className={styles.codePrompt}>$</span>
              <span className={styles.codeCommand}>keylightctl light list</span>
            </div>
            <div className={styles.codeOutput}>
              <div>Key Light Left &nbsp; 80% @ 4500K &nbsp; ON</div>
              <div>Key Light Right &nbsp; 65% @ 5000K &nbsp; ON</div>
            </div>
            <div className={styles.codeLine} style={{marginTop: '1rem'}}>
              <span className={styles.codePrompt}>$</span>
              <span className={styles.codeCommand}>keylightctl group set office brightness 100</span>
            </div>
            <div className={styles.codeOutput}>
              <div>OK</div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

type FeatureItem = {
  icon: string;
  title: string;
  description: string;
};

const features: FeatureItem[] = [
  {
    icon: '\u{1F4A1}',
    title: 'Auto-Discovery',
    description: 'Automatically finds Elgato Key Lights on your network via mDNS/Bonjour. No manual IP configuration needed.',
  },
  {
    icon: '\u{2328}\u{FE0F}',
    title: 'CLI Control',
    description: 'Full control from the command line with keylightctl. Interactive mode, parseable output, and waybar integration.',
  },
  {
    icon: '\u{1F310}',
    title: 'REST API',
    description: 'HTTP API with OpenAPI spec, Bearer token auth, multi-group operations, and real-time WebSocket events.',
  },
  {
    icon: '\u{1F50C}',
    title: 'Unix Socket',
    description: 'Low-latency local control via Unix socket. No auth overhead for same-user processes and scripts.',
  },
  {
    icon: '\u{1F5A5}\u{FE0F}',
    title: 'Desktop Apps',
    description: 'System tray app with CSS theming, GNOME extension, and Waybar module for desktop integration.',
  },
  {
    icon: '\u{1F465}',
    title: 'Group Management',
    description: 'Organize lights into groups for batch control. Set brightness, temperature, and power for all lights at once.',
  },
];

function FeaturesSection() {
  return (
    <section className={styles.features}>
      <div className={styles.featuresInner}>
        <h2 className={styles.featuresTitle}>Control Your Lights, Your Way</h2>
        <div className={styles.featuresGrid}>
          {features.map((feature, idx) => (
            <div key={idx} className={styles.featureCard}>
              <div className={styles.featureIcon}>{feature.icon}</div>
              <h3 className={styles.featureTitle}>{feature.title}</h3>
              <p className={styles.featureDesc}>{feature.description}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

function InstallSection() {
  return (
    <section className={styles.install}>
      <div className={styles.installInner}>
        <h2 className={styles.installTitle}>Quick Install (Arch Linux)</h2>
        <div className={styles.installCode}>
          <span>
            <span className={styles.installPrompt}>$ </span>
            paru -S keylightd-bin
          </span>
        </div>
        <p style={{marginTop: '1rem', color: '#666', fontSize: '0.875rem'}}>
          Also available via <Link to="/docs/getting-started">Homebrew, binary releases, and source builds</Link>
        </p>
      </div>
    </section>
  );
}

export default function Home(): ReactNode {
  const {siteConfig} = useDocusaurusContext();
  return (
    <Layout
      title="Home"
      description={siteConfig.tagline}>
      <HeroSection />
      <FeaturesSection />
      <InstallSection />
    </Layout>
  );
}
